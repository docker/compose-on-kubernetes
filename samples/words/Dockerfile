# BUID
FROM openjdk:8u131-jdk-alpine as build

RUN MAVEN_VERSION=3.5.0 \
 && cd /usr/share \
 && wget http://archive.apache.org/dist/maven/maven-3/$MAVEN_VERSION/binaries/apache-maven-$MAVEN_VERSION-bin.tar.gz -O - | tar xzf - \
 && mv /usr/share/apache-maven-$MAVEN_VERSION /usr/share/maven \
 && ln -s /usr/share/maven/bin/mvn /usr/bin/mvn

WORKDIR /home/lab

COPY pom.xml .
RUN mvn verify -DskipTests --fail-never

COPY src ./src
RUN mvn verify

# RUN
FROM alpine:edge
ENV LANG C.UTF-8
ENV JAVA_HOME /usr/lib/jvm/java-1.8-openjdk/jre
ENV PATH $PATH:/usr/lib/jvm/java-1.8-openjdk/jre/bin:/usr/lib/jvm/java-1.8-openjdk/bin
RUN apk add --no-cache openjdk8-jre="8.151.12-r0" && rm usr/lib/libgif.so.7.0.0 usr/lib/libtasn1.so.6.5.4

ENTRYPOINT ["java", "-Xmx8m", "-Xms8m", "-jar", "./target/words.jar"]
EXPOSE 8080

COPY --from=build /home/lab/target ./target/
